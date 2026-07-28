//go:build darwin || linux

package state

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestReadJSONRejectsSpecialFile(t *testing.T) {
	root, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close state root: %v", err)
		}
	})
	name := "record.fifo"
	if err := syscall.Mkfifo(filepath.Join(root.Path(), name), 0o600); err != nil {
		t.Skipf("create FIFO: %v", err)
	}

	var target map[string]any
	err = root.ReadJSON(name, &target, JSONOptions{Kind: "worktree"})
	if !IsCode(err, CodeInvalid) {
		t.Errorf("error code = %s, expected %s", CodeOf(err), CodeInvalid)
	}
}

func TestOpenReadCandidateDoesNotBlockOnFIFO(t *testing.T) {
	root, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close state root: %v", err)
		}
	})

	name := "candidate.fifo"
	if err := syscall.Mkfifo(filepath.Join(root.Path(), name), 0o600); err != nil {
		t.Skipf("create FIFO: %v", err)
	}

	type openResult struct {
		file *os.File
		err  error
	}
	result := make(chan openResult, 1)
	go func() {
		file, err := openReadCandidate(root.dir, name)
		result <- openResult{file: file, err: err}
	}()

	select {
	case actual := <-result:
		if actual.err != nil {
			t.Fatalf("openReadCandidate returned error: %v", actual.err)
		}
		t.Cleanup(func() {
			if err := actual.file.Close(); err != nil {
				t.Errorf("close FIFO candidate: %v", err)
			}
		})
		info, err := actual.file.Stat()
		if err != nil {
			t.Fatalf("stat FIFO candidate: %v", err)
		}
		expected := os.FileMode(os.ModeNamedPipe)
		actualMode := info.Mode().Type()
		if diff := cmp.Diff(expected, actualMode); diff != "" {
			t.Errorf("mismatch candidate type (-expected, +actual):\n%s", diff)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("openReadCandidate blocked on FIFO")
	}
}
