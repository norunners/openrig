//go:build unix

package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFileNoFollowRejectsFinalSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "runtime.json")
	if err := os.WriteFile(target, []byte("runtime"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	alias := filepath.Join(dir, "operation.json")
	if err := os.Symlink("runtime.json", alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := readFileNoFollow(alias); err == nil {
		t.Fatal("readFileNoFollow error = nil, expected symbolic-link rejection")
	}
	actual, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after rejected operation: %v", err)
	}
	if string(actual) != "runtime" {
		t.Errorf("target content = %q, expected %q", actual, "runtime")
	}
}
