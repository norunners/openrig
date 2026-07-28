//go:build darwin || linux || windows

package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootHandleRemainsAttachedAfterPathReplacement(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "state")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("create state root: %v", err)
	}
	root, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Errorf("close state root: %v", err)
		}
	})

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

	if err := root.dir.WriteFile("runtime.json", []byte("state"), 0o600); err != nil {
		t.Fatalf("write through state root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "runtime.json")); !os.IsNotExist(err) {
		t.Errorf("outside file stat error = %v, expected not exist", err)
	}
	data, err := os.ReadFile(filepath.Join(moved, "runtime.json"))
	if err != nil {
		t.Fatalf("read file beneath moved state root: %v", err)
	}
	if string(data) != "state" {
		t.Errorf("moved state content = %q, expected %q", data, "state")
	}
}
